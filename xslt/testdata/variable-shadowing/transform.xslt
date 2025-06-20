<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>

	<xsl:param name="var" select="'shadow'"/>

	<xsl:template match="/">
		<root>
			<xsl:call-template name="shadow">
				<xsl:with-param name="var" select="'angle'"/>
			</xsl:call-template>
		</root>
	</xsl:template>

	<xsl:template name="shadow">
		<xsl:param name="var"/>
		<xsl:variable name="var" select="'test'"/>
		<item>
			<xsl:value-of select="$var"/>
		</item>
	</xsl:template>
</xsl:stylesheet>