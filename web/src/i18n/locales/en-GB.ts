// English (UK)
import enUS from './en-US'

export default {
  ...enUS,
  localeNames: {
    ...enUS.localeNames,
    'en-GB': 'English (UK)',
    'en-US': 'English (US)',
  },
  // Note: Most British English spelling differences (colour, favourite, etc.)
  // are handled in the UI components or are not used in the codebase.
  // This file can be extended with British English translations as needed.
 as typeof enUS

  personality: {
    title: "Personalities",
    description: "Manage AI assistant personalities",
    create: "Create Personality",
    createNew: "Create new personality",
    createFirst: "Create your first personality",
    name: "Name",
    namePlaceholder: "e.g., Echo, Assistant",
    description: "Description",
    descriptionPlaceholder: "What is this personality for?",
    systemPrompt: "System Prompt",
    systemPromptPlaceholder: "Enter the system prompt for this personality",
    traits: "Traits",
    addTrait: "Add trait",
    traitKey: "Key",
    traitValue: "Value",
    traitWeight: "Weight",
    noPersonalities: "No personalities",
    activate: "Activate",
    active: "Active",
    default: "Default",
    edit: "Edit",
    delete: "Delete",
    confirmDelete: "Are you sure you want to delete this personality?",
    deleteSuccess: "Personality deleted successfully",
    createSuccess: "Personality created successfully",
    updateSuccess: "Personality updated successfully",
    activateSuccess: "Personality activated successfully",
    failedToLoad: "Failed to load personalities",
    failedToCreate: "Failed to create personality",
    failedToUpdate: "Failed to update personality",
    failedToDelete: "Failed to delete personality",
    failedToActivate: "Failed to activate personality",
  },
}
